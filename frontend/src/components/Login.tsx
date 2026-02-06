import React, { useState } from 'react';
import { api } from '../services/api';

interface LoginProps {
  onLogin: (username: string) => void;
}

// 🔥 FRONTEND User (НЕ из wails)
interface User {
  id: string;
  username: string;
  email: string;
  isOnline: boolean;
  status: 'online' | 'away' | 'offline';
  lastSeen?: string;
}

export const Login: React.FC<LoginProps> = ({ onLogin }) => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleLogin = async () => {
    if (!email || !password) {
      alert('Введите email и пароль');
      return;
    }

    setIsLoading(true);
    try {
      
      const user: User = await api.auth.login(email, password) as User;

     
      onLogin(user.username);
    } catch (error: any) {
      alert(`Ошибка входа: ${error.message || error}`);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="login-container">
      <div className="login-box">
        <h1>GoThermo</h1>
        <p>Enterprise Team Chat</p>

        <input
          type="email"
          placeholder="Work Email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className="input-field"
          disabled={isLoading}
        />

        <input
          type="password"
          placeholder="Password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="input-field"
          disabled={isLoading}
          onKeyDown={(e) => e.key === 'Enter' && handleLogin()}
        />

        <button
          onClick={handleLogin}
          className="login-button"
          disabled={isLoading}
        >
          {isLoading ? 'Вход...' : 'Sign In'}
        </button>
      </div>
    </div>
  );
};
