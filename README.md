# GoThermo - Enterprise Team Chat Desktop Application

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)](https://golang.org/)
[![Wails](https://img.shields.io/badge/Wails-DF0024?logo=wails&logoColor=white)](https://wails.io/)
[![React](https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB)](https://reactjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-007ACC?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)

## 📋 Overview

GoThermo is a modern, real-time enterprise team chat desktop application designed to facilitate seamless communication within organizations. Built with Wails (Go + React), GoThermo provides a secure, native, and efficient cross-platform desktop solution for team collaboration.

## ✨ Features

- 💬 **Real-time Messaging**: Instant message delivery with typing indicators
- 🔐 **User Authentication**: Secure login with email/password
- 🖥️ **Native Desktop App**: Cross-platform support (Windows, macOS, Linux)
- 🎨 **Clean UI**: Modern, intuitive interface built with React
- 🏢 **Enterprise Ready**: Built with scalability and security in mind
- 🌍 **Russian & English Support**: Multi-language interface
- ⚡ **High Performance**: Go-powered backend with native performance
- 📴 **Offline Support**: Work offline with local data synchronization
- 🔔 **System Notifications**: Native desktop notifications
- 🚀 **Fast Startup**: Quick launch times with small binary size

## 🛠️ Tech Stack

### Frontend
- React with TypeScript
- React Hooks for state management
- CSS Modules for styling
- Wails Runtime for Go ↔ React communication

### Backend (Go)
- Wails v2 framework
- Native Go performance
- WebSocket for real-time communication
- JWT-based authentication
- Concurrent message processing
- Cross-platform system integration

## 📁 Project Structure
```
GoThermo/
├── build/
│   └── assets/              # Build assets and resources
├── frontend/
│   ├── src/
│   │   ├── assets/          # Images, fonts, static files
│   │   ├── components/      # React components
│   │   │   ├── ChannelModal.tsx
│   │   │   ├── ChannelSidebar.tsx
│   │   │   ├── ChatHeader.tsx
│   │   │   ├── Login.tsx
│   │   │   ├── MessageComposer.tsx
│   │   │   ├── MessageList.tsx
│   │   │   └── UserPanel.tsx
│   │   ├── hooks/           # Custom React hooks
│   │   │   └── useWebSocket.ts
│   │   ├── services/        # API services
│   │   │   └── api.ts
│   │   ├── types/           # TypeScript type definitions
│   │   │   └── index.ts
│   │   ├── App.css          # Main styles
│   │   ├── App.tsx          # Main App component
│   │   ├── main.tsx         # Entry point
│   │   └── style.css        # Global styles
│   ├── index.html
│   ├── package.json
│   ├── package-lock.json
│   ├── tsconfig.json
│   ├── tsconfig.node.json
│   └── vite-env.d.ts
├── wailsjs/                 # Auto-generated Wails bindings
│   ├── go/                  # Go to JS bindings
│   └── runtime/             # Wails runtime
├── go/
│   ├── main/
│   │   ├── app.go           # Main application logic
│   │   └── main.go          # Entry point
│   ├── models/              # Data models
│   ├── runtime/             # Runtime configuration
│   ├── go.mod
│   ├── go.sum
│   ├── wails.json
│   └── wails.go
├── app.go                   # App initialization
├── go.mod                   # Go module file
├── go.sum                   # Go dependencies
├── main.go                  # Application entry
├── models.go                # Data models
├── redis.go                 # Redis integration
├── users.go                 # User management
├── wails.go                 # Wails configuration
├── wails.json               # Wails project config
└── README.md
```

## 🚀 Installation

### Prerequisites

- Go (v1.19 or higher)
- Node.js (v16 or higher)
- npm or yarn
- Git

**Platform-specific requirements:**

**Windows:**
- WebView2 (usually pre-installed on Windows 10/11)
- GCC compiler (MinGW-w64 or TDM-GCC)

**macOS:**
- Xcode Command Line Tools: `xcode-select --install`

**Linux:**
- WebKit2GTK
```bash
# Debian/Ubuntu
sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev

# Fedora
sudo dnf install gtk3-devel webkit2gtk3-devel

# Arch
sudo pacman -S webkit2gtk gtk3
```

### Setup Instructions

1. **Install Wails**
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

2. **Clone the repository**
```bash
git clone https://github.com/yourusername/gothermo.git
cd gothermo
```

3. **Install Go dependencies**
```bash
go mod download
go mod tidy
```

4. **Install frontend dependencies**
```bash
cd frontend
npm install
cd ..
```

5. **Environment Configuration**

Create a `.env` file in the root directory:
```env
# Application
APP_NAME=GoThermo
APP_VERSION=1.0.0
ENVIRONMENT=development

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# Security
JWT_SECRET=your_jwt_secret_here
ENCRYPTION_KEY=your_encryption_key_here

# WebSocket
WS_SERVER_URL=wss://your-server.com/ws
WS_RECONNECT_INTERVAL=5

# Logging
LOG_LEVEL=debug
LOG_FILE=./logs/app.log
```

## 🏃 Running the Application

### Development Mode
```bash
# Run in development mode with hot reload
wails dev

# Or with specific flags
wails dev -browser
```

The application will launch with hot-reload enabled for both frontend and backend.

### Production Build

Build for your current platform:
```bash
# Build for current platform
wails build

# Build with specific flags
wails build -clean -upx -ldflags "-X main.version=1.0.0"
```

Build for specific platforms:
```bash
# Build for Windows
wails build -platform windows/amd64

# Build for macOS (Intel)
wails build -platform darwin/amd64

# Build for macOS (Apple Silicon)
wails build -platform darwin/arm64

# Build for Linux
wails build -platform linux/amd64
```

The compiled binary will be in `build/bin/`.

### Build All Platforms
```bash
# Build for all platforms
wails build -platform windows/amd64,darwin/amd64,darwin/arm64,linux/amd64
```

## 📖 Usage

### First Launch

1. Launch GoThermo application
2. On first run, you'll see the login screen
3. Enter your work email and password
4. Click "Sign In" to access the application

### Starting a Chat

1. Click on a team member or channel from the sidebar
2. Type your message in the MessageComposer
3. Press Enter or click Send

### Creating Channels

1. Click on the "+" button in ChannelSidebar
2. Enter channel name and description
3. Add team members
4. Click Create

### Keyboard Shortcuts

- `Ctrl/Cmd + N` - New chat
- `Ctrl/Cmd + K` - Quick search
- `Ctrl/Cmd + ,` - Settings
- `Ctrl/Cmd + Q` - Quit application
- `Esc` - Close current modal/dialog

## 🔌 Component Structure

### Frontend Components

#### Core Components
```typescript
// Login.tsx - Authentication component
interface LoginProps {
  onLogin: (username: string, password: string) => void;
}

// ChannelSidebar.tsx - Channel list and navigation
interface ChannelSidebarProps {
  channels: Channel[];
  onChannelSelect: (channelId: string) => void;
}

// ChatHeader.tsx - Chat header with channel info
interface ChatHeaderProps {
  channel: Channel;
  onSettingsClick: () => void;
}

// MessageList.tsx - Display chat messages
interface MessageListProps {
  messages: Message[];
  currentUser: User;
}

// MessageComposer.tsx - Message input component
interface MessageComposerProps {
  onSendMessage: (content: string) => void;
  placeholder?: string;
}

// UserPanel.tsx - User profile panel
interface UserPanelProps {
  user: User;
  onLogout: () => void;
}
```

### Backend API Methods

#### User Management
```go
// Login authenticates user
func (a *App) Login(email, password string) (*User, error)

// GetCurrentUser returns logged-in user info
func (a *App) GetCurrentUser() (*User, error)

// UpdateUserStatus updates user online status
func (a *App) UpdateUserStatus(status string) error
```

#### Channel Operations
```go
// GetChannels retrieves all channels
func (a *App) GetChannels() ([]*Channel, error)

// CreateChannel creates a new channel
func (a *App) CreateChannel(name, description string) (*Channel, error)

// JoinChannel adds user to channel
func (a *App) JoinChannel(channelId string) error
```

#### Message Operations
```go
// GetMessages retrieves messages for a channel
func (a *App) GetMessages(channelId string, limit int) ([]*Message, error)

// SendMessage sends a new message
func (a *App) SendMessage(channelId, content string) (*Message, error)

// DeleteMessage deletes a message
func (a *App) DeleteMessage(messageId string) error
```

### WebSocket Integration
```typescript
// useWebSocket.ts - Custom hook for WebSocket
export const useWebSocket = (url: string) => {
  const [isConnected, setIsConnected] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);
  
  // Connect, disconnect, send message handlers
  // Real-time message updates
};
```

## 🤝 Contributing

We welcome contributions to GoThermo! Please follow these steps:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

### Development Guidelines

#### Code Style

**Go**
- Follow Go standard conventions
- Use `gofmt` for code formatting
- Run `go vet` before committing
- Write unit tests for all packages
- Use context for cancellation and timeouts
- Document exported functions and types

**TypeScript/React**
- Use TypeScript for type safety
- Follow React Hooks best practices
- Use functional components
- Implement proper error handling
- Write meaningful commit messages
- Use CSS Modules for component styling

### Testing
```bash
# Run Go tests
go test ./...

# Run with coverage
go test -cover ./...

# Run frontend tests
cd frontend
npm test

# Run Wails doctor to check setup
wails doctor
```

## 🐛 Troubleshooting

### Common Issues

**"Wails command not found"**
```bash
# Ensure GOPATH/bin is in your PATH
export PATH=$PATH:$(go env GOPATH)/bin

# Or reinstall Wails
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

**Build failures on Windows**
- Install MinGW-w64 or TDM-GCC
- Ensure WebView2 is installed
- Run as Administrator if needed

**Build failures on macOS**
```bash
# Install Xcode Command Line Tools
xcode-select --install

# Accept license
sudo xcodebuild -license accept
```

**Build failures on Linux**
```bash
# Install required dependencies
sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev
```

**Redis connection errors**
- Ensure Redis server is running: `redis-cli ping`
- Check REDIS_HOST and REDIS_PORT in .env
- Verify firewall settings

**WebSocket connection issues**
- Check WS_SERVER_URL configuration
- Verify network connectivity
- Check firewall settings

**Application won't start**
- Check logs in `./logs/app.log`
- Run `wails doctor` to diagnose issues
- Verify all dependencies are installed

### Getting Help

- Check the [issues](https://github.com/yourusername/gothermo/issues) section
- Visit [Wails Documentation](https://wails.io/docs/introduction)
- Create a new issue with detailed description
- Include error logs and system information

## 🏗️ Architecture

GoThermo uses a modern desktop application architecture:
```
┌─────────────────────────────────────────┐
│         Frontend (React + TS)           │
│  ┌─────────────────────────────────┐   │
│  │ Components (Login, Chat, etc)   │   │
│  │ Hooks (useWebSocket)            │   │
│  │ Services (API)                  │   │
│  └─────────────────────────────────┘   │
└──────────────┬──────────────────────────┘
               │ Wails Runtime
┌──────────────┴──────────────────────────┐
│         Backend (Go)                    │
│  ┌─────────────────────────────────┐   │
│  │ App Logic (app.go, main.go)     │   │
│  │ Models (User, Channel, Message) │   │
│  │ WebSocket Hub                   │   │
│  │ Redis Integration               │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

### Data Flow

1. User interacts with React UI
2. Component calls Wails-bound Go function
3. Go processes request (auth, data fetch, etc.)
4. Redis stores/retrieves data
5. WebSocket pushes real-time updates
6. UI updates automatically

## 📊 Performance

- Native performance with Go backend
- Small binary size (~20-30MB compressed)
- Fast startup time (<1 second)
- Low memory footprint (~50-100MB)
- Efficient concurrent message processing
- Redis for fast data access
- Local-first architecture for offline support

## 📦 Distribution

### Creating Installers

**Windows (NSIS):**
```bash
wails build -nsis
```

**macOS (DMG):**
```bash
wails build -dmg
```

**Linux (AppImage/DEB/RPM):**
```bash
# AppImage
wails build -appimage

# Debian package
wails build -deb

# RPM package
wails build -rpm
```

### Code Signing

**macOS:**
```bash
wails build -codesign "Developer ID Application: Your Name"
```

**Windows:**
```bash
# Use SignTool after build
signtool sign /f cert.pfx /p password build/bin/GoThermo.exe
```

## 🔐 Security

- JWT-based authentication
- Secure password hashing
- Secure WebSocket connections (WSS)
- Redis for session management
- No credentials stored in plain text
- Regular security updates

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Wails](https://wails.io/) - Amazing Go + Web framework
- [Redis](https://redis.io/) - In-memory data store
- Built with modern web and native technologies
- Inspired by Slack, Discord, and Microsoft Teams
- Thanks to all contributors and testers
- Go and React communities

## 📧 Contact

**Project Maintainer:** [Max Chernikov]

- Email: maksikos973@gmail.com
- GitHub: [@Maksikos-ctrl](https://github.com/Maksikos-ctrl)
- Website: https://maksymchernikovportfolio.vercel.app/

## 🗺️ Roadmap

- [x] Basic authentication and login
- [x] Real-time messaging with WebSocket
- [x] Channel management
- [x] User presence indicators
- [ ] End-to-end encryption
- [ ] Voice and video calls
- [ ] Screen sharing
- [ ] File sharing with drag & drop
- [ ] Message search and history
- [ ] Custom emoji and reactions
- [ ] Thread replies
- [ ] Mobile companion app
- [ ] Plugin system
- [ ] Custom themes
- [ ] Integration with popular tools (Jira, GitHub, etc.)

---

**Happy collaborating with GoThermo!** 🚀💬