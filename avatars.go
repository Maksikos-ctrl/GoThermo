package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"strings"
)

// Profile pictures.
//
// The app resizes the chosen picture, but the server never trusts that: it
// decodes whatever arrives, cuts the middle square, scales it to avatarSize and
// writes a fresh JPEG itself. What is stored (and sent to everyone as a data
// URL) is therefore always a small JPEG made here, whatever was uploaded.
//
// Storage: the Redis hash "avatars", lowercase username -> data URL.

const (
	avatarSize       = 128
	maxAvatarBytes   = 2 << 20 // what the upload may weigh
	maxAvatarPixels  = 24_000_000
	avatarsKey       = "avatars"
	avatarDataPrefix = "data:image/jpeg;base64,"
)

// squareThumb cuts the middle square of img and scales it to size x size with
// a box filter (every output pixel is the average of the source area it covers).
// Transparent areas turn white.
func squareThumb(img image.Image, size int) *image.RGBA {
	b := img.Bounds()
	side := b.Dx()
	if b.Dy() < side {
		side = b.Dy()
	}
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2

	// flatten onto white so that PNG transparency does not become black
	flat := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(flat, flat.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), img, image.Point{X: x0, Y: y0}, draw.Over)

	out := image.NewRGBA(image.Rect(0, 0, size, size))
	for oy := 0; oy < size; oy++ {
		sy0, sy1 := oy*side/size, (oy+1)*side/size
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for ox := 0; ox < size; ox++ {
			sx0, sx1 := ox*side/size, (ox+1)*side/size
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var r, g, bl, n uint32
			for y := sy0; y < sy1 && y < side; y++ {
				row := flat.Pix[y*flat.Stride:]
				for x := sx0; x < sx1 && x < side; x++ {
					r += uint32(row[x*4])
					g += uint32(row[x*4+1])
					bl += uint32(row[x*4+2])
					n++
				}
			}
			if n == 0 {
				n = 1
			}
			o := out.PixOffset(ox, oy)
			out.Pix[o], out.Pix[o+1], out.Pix[o+2], out.Pix[o+3] = uint8(r/n), uint8(g/n), uint8(bl/n), 255
		}
	}
	return out
}

// makeAvatar turns uploaded picture bytes into the stored data URL.
func makeAvatar(data []byte) (string, error) {
	if len(data) == 0 || len(data) > maxAvatarBytes {
		return "", fmt.Errorf("the picture must be smaller than %d MB", maxAvatarBytes>>20)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("that file is not a picture (use JPEG, PNG or GIF)")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > maxAvatarPixels {
		return "", fmt.Errorf("that picture is too large")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("that file is not a picture (use JPEG, PNG or GIF)")
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, squareThumb(img, avatarSize), &jpeg.Options{Quality: 85}); err != nil {
		return "", internalFailure("avatar encode", err)
	}
	return avatarDataPrefix + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// SetAvatar saves the signed-in person's profile picture. base64Data may be a
// bare base64 string or a data URL.
func (a *App) SetAvatar(token, user, base64Data string) error {
	if err := requireUser(token, user); err != nil {
		return err
	}
	if i := strings.Index(base64Data, ","); i != -1 && strings.HasPrefix(base64Data, "data:") {
		base64Data = base64Data[i+1:]
	}
	if len(base64Data) > maxAvatarBytes/3*4+64 {
		return fmt.Errorf("the picture must be smaller than %d MB", maxAvatarBytes>>20)
	}
	raw, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("that file is not a picture (use JPEG, PNG or GIF)")
	}
	url, err := makeAvatar(raw)
	if err != nil {
		return err
	}
	if err := redisClient.HSet(ctx, avatarsKey, strings.ToLower(user), url).Err(); err != nil {
		return internalFailure("avatar save", err)
	}
	a.announceAvatar(user, url)
	return nil
}

// RemoveAvatar goes back to the coloured letter.
func (a *App) RemoveAvatar(token, user string) error {
	if err := requireUser(token, user); err != nil {
		return err
	}
	if err := redisClient.HDel(ctx, avatarsKey, strings.ToLower(user)).Err(); err != nil {
		return internalFailure("avatar remove", err)
	}
	a.announceAvatar(user, "")
	return nil
}

func (a *App) announceAvatar(user, url string) {
	if a.hub != nil {
		a.hub.BroadcastAvatarChanged(user, url)
	}
}

// withAvatars adds each person's picture (if any) to a copy of the user list.
func withAvatars(users []User) []User {
	all, err := redisClient.HGetAll(ctx, avatarsKey).Result()
	if err != nil || len(all) == 0 {
		return users
	}
	out := make([]User, len(users))
	for i, u := range users {
		u.Avatar = all[strings.ToLower(u.Username)]
		out[i] = u
	}
	return out
}

// AvatarChanged is the live event that tells everyone a picture changed.
type AvatarChanged struct {
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

func (h *Hub) BroadcastAvatarChanged(username, avatar string) {
	data, err := json.Marshal(WSMessage{Type: "avatar_changed", Payload: AvatarChanged{Username: username, Avatar: avatar}})
	if err != nil {
		return
	}
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	for _, client := range h.clients {
		select {
		case client.Send <- data:
		default: // a slow client simply misses it; the next user list has the picture
		}
	}
}
