import React, { useState } from 'react';
import { api } from '../services/api';

interface LoginProps {
  onLogin: (username: string) => void;
}

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
  const [confirmPassword, setConfirmPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [isSignUp, setIsSignUp] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const validateEmail = (email: string): string | null => {
    if (!email) return 'Enter your email';
    if (!email.includes('@')) return 'Invalid email format';
    if (email.length < 5) return 'Email is too short';
    return null;
  };

  const validatePassword = (password: string): string | null => {
    if (!password) return 'Enter your password';
    if (password.length < 4) return 'Password must be at least 4 characters';
    return null;
  };

  const handleLogin = async () => {
    setError('');
    setSuccess('');

    const emailError = validateEmail(email);
    if (emailError) {
      setError(emailError);
      return;
    }

    const passwordError = validatePassword(password);
    if (passwordError) {
      setError(passwordError);
      return;
    }

    setIsLoading(true);
    try {
      const user: User = await api.auth.login(email, password) as User;
      setSuccess(`✅ Welcome back, ${user.username}!`);
      setTimeout(() => onLogin(user.username), 1000);
    } catch (error: any) {
      const errorMessage = error.message || error.toString();
      
      if (errorMessage.includes('не найден') || errorMessage.includes('not found')) {
        setError('❌ User not found');
      } else if (errorMessage.includes('неверный пароль') || errorMessage.includes('invalid password')) {
        setError('❌ Invalid password');
      } else if (errorMessage.includes('уже вошли')) {
        setError('❌ You are already logged in from another device');
      } else {
        setError(`❌ ${errorMessage}`);
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleRegister = async () => {
    setError('');
    setSuccess('');

    const emailError = validateEmail(email);
    if (emailError) {
      setError(emailError);
      return;
    }

    const passwordError = validatePassword(password);
    if (passwordError) {
      setError(passwordError);
      return;
    }

    if (password !== confirmPassword) {
      setError('❌ Passwords do not match');
      return;
    }

    setIsLoading(true);
    try {
      const user: User = await api.auth.register(email, password) as User;
      setSuccess(`✅ Account created successfully! Welcome, ${user.username}!`);
      setTimeout(() => onLogin(user.username), 1500);
    } catch (error: any) {
      const errorMessage = error.message || error.toString();
      
      if (errorMessage.includes('уже существует')) {
        setError('❌ User with this email already exists');
      } else {
        setError(`❌ ${errorMessage}`);
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleSubmit = () => {
    if (isSignUp) {
      handleRegister();
    } else {
      handleLogin();
    }
  };

  const switchMode = () => {
    setIsSignUp(!isSignUp);
    setError('');
    setSuccess('');
    setPassword('');
    setConfirmPassword('');
  };

  return (
    <div className="login-container">
      <div className="login-box">
        <h1>GoThermo</h1>
        <p>Enterprise Team Chat</p>

        <div style={{
          display: 'flex',
          gap: '10px',
          marginBottom: '20px',
          borderBottom: '2px solid #e0e0e0'
        }}>
          <button
            onClick={() => !isLoading && switchMode()}
            disabled={isLoading}
            style={{
              flex: 1,
              padding: '12px',
              border: 'none',
              background: 'none',
              cursor: isLoading ? 'not-allowed' : 'pointer',
              borderBottom: !isSignUp ? '3px solid #007bff' : 'none',
              color: !isSignUp ? '#007bff' : '#666',
              fontWeight: !isSignUp ? 'bold' : 'normal',
              fontSize: '16px'
            }}
          >
            Sign In
          </button>
          <button
            onClick={() => !isLoading && switchMode()}
            disabled={isLoading}
            style={{
              flex: 1,
              padding: '12px',
              border: 'none',
              background: 'none',
              cursor: isLoading ? 'not-allowed' : 'pointer',
              borderBottom: isSignUp ? '3px solid #007bff' : 'none',
              color: isSignUp ? '#007bff' : '#666',
              fontWeight: isSignUp ? 'bold' : 'normal',
              fontSize: '16px'
            }}
          >
            Sign Up
          </button>
        </div>

        {success && (
          <div style={{
            padding: '12px',
            marginBottom: '15px',
            background: '#e8f5e9',
            border: '1px solid #c8e6c9',
            borderRadius: '4px',
            color: '#2e7d32',
            fontSize: '14px',
            display: 'flex',
            alignItems: 'center',
            gap: '8px'
          }}>
            <span style={{ fontSize: '20px' }}>🎉</span>
            <span>{success}</span>
          </div>
        )}

        {error && (
          <div style={{
            padding: '12px',
            marginBottom: '15px',
            background: '#ffebee',
            border: '1px solid #ffcdd2',
            borderRadius: '4px',
            color: '#c62828',
            fontSize: '14px',
            display: 'flex',
            alignItems: 'center',
            gap: '8px'
          }}>
            <span>⚠️</span>
            <span>{error}</span>
          </div>
        )}

        <input
          type="email"
          placeholder="Work Email"
          value={email}
          onChange={(e) => {
            setEmail(e.target.value);
            setError('');
            setSuccess('');
          }}
          className="input-field"
          disabled={isLoading}
          autoComplete="email"
        />

        <input
          type="password"
          placeholder="Password"
          value={password}
          onChange={(e) => {
            setPassword(e.target.value);
            setError('');
            setSuccess('');
          }}
          className="input-field"
          disabled={isLoading}
          autoComplete={isSignUp ? 'new-password' : 'current-password'}
          onKeyDown={(e) => e.key === 'Enter' && !isSignUp && handleSubmit()}
        />

        {isSignUp && (
          <>
            <input
              type="password"
              placeholder="Confirm Password"
              value={confirmPassword}
              onChange={(e) => {
                setConfirmPassword(e.target.value);
                setError('');
                setSuccess('');
              }}
              className="input-field"
              disabled={isLoading}
              autoComplete="new-password"
              onKeyDown={(e) => e.key === 'Enter' && handleSubmit()}
            />
            
            <div style={{
              fontSize: '12px',
              color: '#666',
              marginTop: '-10px',
              marginBottom: '10px',
              textAlign: 'left'
            }}>
              💡 Minimum 4 characters
            </div>
          </>
        )}

        <button
          onClick={handleSubmit}
          className="login-button"
          disabled={isLoading}
          style={{
            opacity: isLoading ? 0.7 : 1,
            cursor: isLoading ? 'not-allowed' : 'pointer'
          }}
        >
          {isLoading ? (
            <span style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px' }}>
              <span className="spinner"></span>
              {isSignUp ? 'Creating account...' : 'Signing in...'}
            </span>
          ) : (
            isSignUp ? 'Sign Up' : 'Sign In'
          )}
        </button>

        <div style={{
          marginTop: '20px',
          fontSize: '13px',
          color: '#888'
        }}>
          {isSignUp ? (
            <>
              Already have an account?{' '}
              <button
                onClick={switchMode}
                disabled={isLoading}
                style={{
                  background: 'none',
                  border: 'none',
                  color: '#007bff',
                  cursor: 'pointer',
                  textDecoration: 'underline',
                  fontSize: '13px'
                }}
              >
                Sign In
              </button>
            </>
          ) : (
            <>
              Don't have an account?{' '}
              <button
                onClick={switchMode}
                disabled={isLoading}
                style={{
                  background: 'none',
                  border: 'none',
                  color: '#007bff',
                  cursor: 'pointer',
                  textDecoration: 'underline',
                  fontSize: '13px'
                }}
              >
                Sign Up
              </button>
            </>
          )}
        </div>
      </div>

      <style>{`
        .spinner {
          display: inline-block;
          width: 16px;
          height: 16px;
          border: 2px solid rgba(255,255,255,0.3);
          border-radius: 50%;
          border-top-color: white;
          animation: spin 0.8s linear infinite;
        }
        
        @keyframes spin {
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
};