# GoThermo - Enterprise Team Chat Application

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![TypeScript](https://img.shields.io/badge/TypeScript-007ACC?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)
[![React](https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB)](https://reactjs.org/)
[![Node.js](https://img.shields.io/badge/Node.js-339933?logo=node.js&logoColor=white)](https://nodejs.org/)
[![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)](https://golang.org/)

## 📋 Overview

GoThermo is a modern, real-time enterprise team chat application designed to facilitate seamless communication within organizations. Built with a React frontend and a robust backend, GoThermo provides a secure and efficient platform for team collaboration.

## ✨ Features

- 💬 **Real-time Messaging**: Instant message delivery with typing indicators
- 🔐 **User Authentication**: Secure login with email/password
- 📱 **Responsive Design**: Works on desktop and mobile devices
- 🎨 **Clean UI**: Modern, intuitive interface
- 🏢 **Enterprise Ready**: Built with scalability and security in mind
- 🌍 **Russian & English Support**: Multi-language interface

## 🛠️ Tech Stack

### Frontend
- React with TypeScript
- React Hooks for state management
- CSS Modules for styling
- Axios/Fetch API for HTTP requests

### Backend
- Node.js with Express
- TypeScript for type safety
- Authentication (JWT-based)
- WebSocket for real-time communication

## 📁 Project Structure
```
GoThermo/
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   │   ├── Login.tsx
│   │   │   └── ...
│   │   ├── services/
│   │   │   └── api.ts
│   │   └── App.tsx
├── backend/
│   ├── src/
│   │   ├── routes/
│   │   ├── models/
│   │   └── server.ts
└── README.md
```

## 🚀 Installation

### Prerequisites

- Node.js (v14 or higher)
- npm or yarn
- Git

### Setup Instructions

1. **Clone the repository**
```bash
git clone https://github.com/yourusername/gothermo.git
cd gothermo
```

2. **Install dependencies for frontend**
```bash
cd frontend
npm install
```

3. **Install dependencies for backend**
```bash
cd ../backend
npm install
```

4. **Environment Configuration**

Create `.env` files in both frontend and backend directories:

**Frontend (`.env`)**
```env
REACT_APP_API_URL=http://localhost:5000
```

**Backend (`.env`)**
```env
PORT=5000
JWT_SECRET=your_jwt_secret_here
DATABASE_URL=your_database_url
```

## 🏃 Running the Application

### Development Mode

1. **Start the backend server**
```bash
cd backend
npm run dev
```

2. **Start the frontend development server**
```bash
cd frontend
npm start
```

3. **Access the application**
   - Frontend: http://localhost:3000
   - Backend API: http://localhost:5000

### Production Build
```bash
# Build frontend
cd frontend
npm run build

# Start backend in production
cd ../backend
npm start
```

## 📖 Usage

### Registration/Login
1. Navigate to the login page
2. Enter your work email and password
3. Click "Sign In"

### Starting a Chat
1. Select a team member from the sidebar
2. Type your message and press Enter

### Creating Channels
1. Click on "New Channel" button
2. Add members and set channel name

## 🔌 API Documentation

### Authentication Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/login` | User login |
| POST | `/api/auth/register` | User registration |
| POST | `/api/auth/logout` | User logout |

### Chat Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/chats` | Get user chats |
| POST | `/api/chats` | Create new chat |
| GET | `/api/chats/:id/messages` | Get chat messages |
| POST | `/api/chats/:id/messages` | Send message |

## 🤝 Contributing

We welcome contributions to GoThermo! Please follow these steps:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

### Development Guidelines

#### Code Style
- Use TypeScript for type safety
- Follow React Hooks best practices
- Use functional components
- Implement proper error handling
- Write meaningful commit messages

### Testing
```bash
# Run frontend tests
cd frontend
npm test

# Run backend tests
cd backend
npm test
```

## 🐛 Troubleshooting

### Common Issues

**"Port already in use" error**
- Change port in `.env` file
- Or kill the process using the port

**CORS errors**
- Ensure backend CORS configuration includes frontend URL
- Check environment variables

**Authentication issues**
- Verify JWT secret matches between frontend and backend
- Clear browser cookies and localStorage

### Getting Help

- Check the [issues](https://github.com/yourusername/gothermo/issues) section for existing problems
- Create a new issue with detailed description
- Include error messages and steps to reproduce

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with modern web technologies
- Inspired by popular team collaboration tools
- Thanks to all contributors and testers

## 📧 Contact

**Project Maintainer:** [Your Name]

- Email: your.email@example.com
- GitHub: [@yourusername](https://github.com/yourusername)

---

**Happy collaborating with GoThermo!** 🚀