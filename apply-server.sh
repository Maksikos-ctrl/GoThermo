#!/bin/bash
# Run from the project root. Makes small, checked edits to existing files.
set -e
fail() { echo "❌ $1"; exit 1; }

# 1. main.go: the desktop entry point must not be part of the server build
head -1 main.go | grep -q 'go:build !server' || perl -0pi -e '$_ = "//go:build !server\n\n" . $_' main.go
head -1 main.go | grep -q 'go:build !server' || fail "main.go"

# 2. redis.go: Redis address and password from the environment
perl -pi -e 's/Addr:(\s+)"localhost:6379",/Addr:$1envOr("REDIS_ADDR", "localhost:6379"),/; s/Password:(\s+)"",/Password:$1envOr("REDIS_PASSWORD", ""),/' redis.go
grep -q 'envOr("REDIS_ADDR"' redis.go || fail "redis.go (Addr line not found)"
grep -q 'envOr("REDIS_PASSWORD"' redis.go || fail "redis.go (Password line not found)"

# 3. frontend: all backend calls go through the transport
perl -pi -e "s#from '\.\./\.\./wailsjs/go/main/App';#from './transport';#" frontend/src/services/api.ts
grep -q "from './transport'" frontend/src/services/api.ts || fail "api.ts"

# 4. frontend: WebSocket address comes from the chosen server
perl -0pi -e 's/    const protocol = window\.location\.protocol === .https:. \? .wss:. : .ws:.;\n    const wsUrl = `\$\{protocol\}\/\/127\.0\.0\.1:8081\/ws\?token=\$\{encodeURIComponent\(token\)\}`;\n\n    const socket = new WebSocket\(wsUrl\);/    const socket = new WebSocket(wsUrl(token));/' frontend/src/hooks/useWebSocket.ts
grep -q 'new WebSocket(wsUrl(token))' frontend/src/hooks/useWebSocket.ts || fail "useWebSocket.ts"
grep -q "services/server'" frontend/src/hooks/useWebSocket.ts || perl -0pi -e "s#(import \{ getAuthToken \} from '\.\./services/session';\n)#\$1import { wsUrl } from '../services/server';\n#" frontend/src/hooks/useWebSocket.ts
grep -q "services/server'" frontend/src/hooks/useWebSocket.ts || fail "useWebSocket.ts import"

# 5. frontend: server picker on the sign-in screen
grep -q ServerPicker frontend/src/App.tsx || perl -0pi -e "s#(import \{ Login \} from './components/Login';\n)#\$1import { ServerPicker } from './components/ServerPicker';\n#; s#return <Login onLogin=\{handleLogin\} />;#return (\n      <>\n        <Login onLogin={handleLogin} />\n        <ServerPicker />\n      </>\n    );#" frontend/src/App.tsx
grep -q '<ServerPicker />' frontend/src/App.tsx || fail "App.tsx"

echo "✅ all edits applied"
