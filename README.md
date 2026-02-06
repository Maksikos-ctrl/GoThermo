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
- SQLite/PostgreSQL for data storage
- Gorilla WebSocket for real-time communication
- JWT-based authentication
- Concurrent message processing
- Cross-platform system integration

## 📁 Project Structure
```
GoThermo/
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   │   ├── Login.tsx
│   │   │   ├── ChatWindow.tsx
│   │   │   ├── Sidebar.tsx
│   │   │   └── ...
│   │   ├── services/
│   │   │   └── wailsApi.ts
│   │   ├── styles/
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── package.json
│   └── wailsjs/
│       ├── go/
│       └── runtime/
├── backend/
│   ├── app.go
│   ├── main.go
│   ├── internal/
│   │   ├── auth/
│   │   │   └── auth.go
│   │   ├── chat/
│   │   │   ├── chat.go
│   │   │   └── message.go
│   │   ├── database/
│   │   │   └── db.go
│   │   ├── models/
│   │   │   ├── user.go
│   │   │   ├── chat.go
│   │   │   └── message.go
│   │   └── websocket/
│   │       └── hub.go
│   ├── pkg/
│   │   ├── config/
│   │   └── utils/
│   └── go.mod
├── build/
│   ├── windows/
│   ├── darwin/
│   └── linux/
├── wails.json
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

# Database
DATABASE_PATH=./data/gothermo.db
# Or for PostgreSQL
# DATABASE_URL=postgres://user:password@localhost:5432/gothermo

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
2. On first run, you'll see the welcome screen
3. Sign in with your work email and password
4. Or register for a new account

### Starting a Chat

1. Click on a team member from the sidebar
2. Type your message in the input field
3. Press Enter or click Send

### Creating Channels

1. Click on "New Channel" button
2. Enter channel name
3. Add team members
4. Click Create

### Keyboard Shortcuts

- `Ctrl/Cmd + N` - New chat
- `Ctrl/Cmd + K` - Quick search
- `Ctrl/Cmd + ,` - Settings
- `Ctrl/Cmd + Q` - Quit application
- `Esc` - Close current dialog

## 🔌 API Documentation

### Go Backend Methods (Exposed to Frontend)

#### Authentication
```go
// Login authenticates user
func (a *App) Login(email, password string) (*User, error)

// Register creates new user account
func (a *App) Register(email, password, name string) (*User, error)

// Logout ends user session
func (a *App) Logout() error
```

#### Chat Operations
```go
// GetChats retrieves all user chats
func (a *App) GetChats() ([]*Chat, error)

// CreateChat creates a new chat
func (a *App) CreateChat(name string, participants []string) (*Chat, error)

// GetMessages retrieves messages for a chat
func (a *App) GetMessages(chatID string, limit int) ([]*Message, error)

// SendMessage sends a new message
func (a *App) SendMessage(chatID, content string) (*Message, error)
```

#### User Operations
```go
// GetCurrentUser returns current logged-in user
func (a *App) GetCurrentUser() (*User, error)

// GetUsers retrieves all users
func (a *App) GetUsers() ([]*User, error)

// UpdateProfile updates user profile
func (a *App) UpdateProfile(name, avatar string) error
```

### Frontend Usage Example
```typescript
import { Login, GetChats, SendMessage } from '../wailsjs/go/main/App';

// Login
const user = await Login(email, password);

// Get chats
const chats = await GetChats();

// Send message
const message = await SendMessage(chatId, content);
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

### Building Documentation
```bash
# Generate Go documentation
godoc -http=:6060

# Generate API documentation
wails generate module
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

**Database connection errors**
- Check DATABASE_PATH or DATABASE_URL in .env
- Ensure data directory exists and has write permissions
- Verify SQLite/PostgreSQL is properly installed

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
- Join our community Discord
- Create a new issue with detailed description

## 🏗️ Architecture

GoThermo uses a modern desktop application architecture:

- **Frontend (React + TypeScript)**: User interface with Wails runtime integration
- **Backend (Go)**: Business logic, database operations, WebSocket management
- **Communication**: Direct Go ↔ React binding via Wails runtime
- **Data Storage**: SQLite for local storage, optional PostgreSQL for enterprise
- **Real-time**: WebSocket connections for live updates

### Data Flow
```
User Interface (React)
        ↓↑
   Wails Runtime
        ↓↑
  Go Backend (App)
        ↓↑
   Database Layer
```

## 📊 Performance

- Native performance with Go backend
- Small binary size (~20-30MB compressed)
- Fast startup time (<1 second)
- Low memory footprint
- Efficient concurrent message processing
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
- Encrypted local database
- Secure WebSocket connections (WSS)
- No credentials stored in plain text
- Regular security updates

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Wails](https://wails.io/) - Amazing Go + Web framework
- Built with modern web and native technologies
- Inspired by popular team collaboration tools
- Thanks to all contributors and testers
- Go and React communities

## 📧 Contact

**Project Maintainer:** [Your Name]

- Email: your.email@example.com
- GitHub: [@yourusername](https://github.com/yourusername)
- Website: https://gothermo.io

## 🗺️ Roadmap

- [ ] End-to-end encryption
- [ ] Voice and video calls
- [ ] Screen sharing
- [ ] File sharing with drag & drop
- [ ] Mobile companion app
- [ ] Plugin system
- [ ] Custom themes
- [ ] Integration with popular tools (Jira, GitHub, etc.)

---

**Happy collaborating with GoThermo!** 🚀💬