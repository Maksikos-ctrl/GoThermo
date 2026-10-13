import React, { useState } from 'react';
import { api } from '../services/api';
import { setAuthToken } from '../services/session';

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
  token?: string;
}

export const Login: React.FC<LoginProps> = ({ onLogin }) => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [isSignUp, setIsSignUp] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [googleWaiting, setGoogleWaiting] = useState(false);

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
      setAuthToken(user.token || '');
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
      setAuthToken(user.token || '');
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

  const handleGoogleLogin = async () => {
    setError('');
    setSuccess('');
    setIsLoading(true);
    setGoogleWaiting(true);
    try {
      const user: User = await api.auth.loginWithGoogle(isSignUp) as User;
      setAuthToken(user.token || '');
      setSuccess(`✅ Welcome, ${user.username}!`);
      setTimeout(() => onLogin(user.username), 800);
    } catch (error: any) {
      const errorMessage = error?.message || error?.toString() || 'Google sign-in failed';
      if (!errorMessage.includes('cancelled')) {
        setError(`❌ ${errorMessage}`);
      }
    } finally {
      setIsLoading(false);
      setGoogleWaiting(false);
    }
  };

  const handleCancelGoogle = () => {
    api.auth.cancelGoogleLogin();
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
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          margin: '18px 0 14px',
          color: '#999',
          fontSize: '12px'
        }}>
          <div style={{ flex: 1, height: '1px', background: '#e0e0e0' }} />
          <span>or</span>
          <div style={{ flex: 1, height: '1px', background: '#e0e0e0' }} />
        </div>

        {googleWaiting ? (
          <div style={{
            padding: '12px',
            border: '1px solid #d0d7de',
            borderRadius: '4px',
            fontSize: '13px',
            color: '#555',
            textAlign: 'center'
          }}>
            <div style={{ marginBottom: '8px' }}>
              🌐 Finish signing in with Google in your browser…
            </div>
            <button
              onClick={handleCancelGoogle}
              style={{
                background: 'none',
                border: 'none',
                color: '#007bff',
                cursor: 'pointer',
                textDecoration: 'underline',
                fontSize: '13px'
              }}
            >
              Cancel
            </button>
          </div>
        ) : (
          <button
            onClick={handleGoogleLogin}
            disabled={isLoading}
            style={{
              width: '100%',
              padding: '11px',
              border: '1px solid #d0d7de',
              borderRadius: '4px',
              background: '#fff',
              color: '#333',
              fontSize: '14px',
              fontWeight: 500,
              cursor: isLoading ? 'not-allowed' : 'pointer',
              opacity: isLoading ? 0.7 : 1,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '10px'
            }}
          >
            <svg width="18" height="18" viewBox="0 0 48 48" aria-hidden="true">
              <path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"/>
              <path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"/>
              <path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"/>
              <path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"/>
            </svg>
            {isSignUp ? 'Sign up with Google' : 'Sign in with Google'}
          </button>
        )}

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