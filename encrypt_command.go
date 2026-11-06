package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// `gothermo-server encrypt` seals the text of messages that were stored before
// the data key was set; `-decrypt` turns sealed text back into plain text (to
// switch encryption off for good). Both are safe to run again and again: rows
// are visited once, in order, and rows that need no change are left alone.
func runEncryptCommand(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("encrypt", flag.ContinueOnError)
	fs.SetOutput(out)
	decrypt := fs.Bool("decrypt", false, "turn sealed messages back into plain text")
	dryRun := fs.Bool("dry-run", false, "only count what would change")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := loadDataKey(); err != nil {
		fmt.Fprintln(out, "❌", err)
		return 1
	}
	if !encryptionOn() {
		fmt.Fprintln(out, "❌ GOTHERMO_DATA_KEY is not set: there is no key to work with.")
		return 1
	}
	pool, err := openMigrationTarget()
	if err != nil {
		fmt.Fprintln(out, "❌", err)
		return 1
	}
	defer pool.Close()
	n, err := convertMessageText(pool, !*decrypt, *dryRun)
	verb := map[bool]string{true: "sealed", false: "turned back to plain text"}[!*decrypt]
	if *dryRun {
		verb = "would be changed"
	}
	if err != nil {
		fmt.Fprintf(out, "❌ stopped after %d messages: %v\n", n, err)
		return 1
	}
	fmt.Fprintf(out, "✓ %d messages %s.\n", n, verb)
	return 0
}

// convertMessageText walks the messages table in order and rewrites the body
// of every row that needs it. It returns how many rows were (or would be) changed.
func convertMessageText(pool *pgxpool.Pool, encrypt, dryRun bool) (int, error) {
	const batchSize = 500
	var (
		total int
		last  int64 = -1
	)
	for {
		rows, err := pool.Query(ctx,
			"SELECT seq, id, body FROM messages WHERE seq > $1 ORDER BY seq LIMIT $2", last, batchSize)
		if err != nil {
			return total, err
		}
		type row struct {
			seq      int64
			id, body string
		}
		var batch []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.seq, &r.id, &r.body); err != nil {
				rows.Close()
				return total, err
			}
			batch = append(batch, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return total, err
		}
		if len(batch) == 0 {
			return total, nil
		}
		last = batch[len(batch)-1].seq

		type change struct{ id, body string }
		var changes []change
		for _, r := range batch {
			sealed := strings.HasPrefix(r.body, encPrefix)
			switch {
			case r.body == "":
				continue
			case encrypt && sealed, !encrypt && !sealed:
				continue
			case encrypt:
				changes = append(changes, change{r.id, sealText(r.body)})
			default:
				plain := openText(r.body)
				if plain == badKeyNotice || plain == noKeyNotice {
					return total, fmt.Errorf("message %s cannot be decrypted with this key", r.id)
				}
				changes = append(changes, change{r.id, plain})
			}
		}
		if len(changes) == 0 {
			continue
		}
		if dryRun {
			total += len(changes)
			continue
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return total, err
		}
		for _, c := range changes {
			if _, err := tx.Exec(ctx, "UPDATE messages SET body = $2 WHERE id = $1", c.id, c.body); err != nil {
				tx.Rollback(ctx)
				return total, err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return total, err
		}
		total += len(changes)
	}
}
