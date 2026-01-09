import { useState, useCallback } from 'react';
import { MVChat2Client } from '../client';
import { User, LoginCredentials, SignupData, AuthResult } from '../types';

export interface UseAuthResult {
  isAuthenticated: boolean;
  user: User | null;
  userID: string | null;
  mustChangePassword: boolean;
  emailVerified: boolean;
  isLoading: boolean;
  error: Error | null;
  login: (credentials: LoginCredentials) => Promise<AuthResult>;
  loginWithToken: (token: string) => Promise<AuthResult>;
  signup: (data: SignupData) => Promise<AuthResult>;
  logout: () => void;
  changePassword: (oldPassword: string, newPassword: string) => Promise<void>;
  updateProfile: (profile: any) => Promise<void>;
  updatePrivateData: (data: any) => Promise<void>;
  updateEmail: (email: string) => Promise<void>;
}

export function useAuth(client: MVChat2Client): UseAuthResult {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const login = useCallback(async (credentials: LoginCredentials) => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await client.login(credentials);
      return result;
    } catch (err) {
      setError(err as Error);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [client]);

  const loginWithToken = useCallback(async (token: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await client.loginWithToken(token);
      return result;
    } catch (err) {
      setError(err as Error);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [client]);

  const signup = useCallback(async (data: SignupData) => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await client.signup(data);
      return result;
    } catch (err) {
      setError(err as Error);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [client]);

  const logout = useCallback(() => {
    client.logout();
  }, [client]);

  const changePassword = useCallback(async (oldPassword: string, newPassword: string) => {
    setIsLoading(true);
    setError(null);
    try {
      await client.changePassword({ oldPassword, newPassword });
    } catch (err) {
      setError(err as Error);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [client]);

  const updateProfile = useCallback(async (profile: any) => {
    setIsLoading(true);
    setError(null);
    try {
      await client.updateProfile(profile);
    } catch (err) {
      setError(err as Error);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [client]);

  const updatePrivateData = useCallback(async (data: any) => {
    setIsLoading(true);
    setError(null);
    try {
      await client.updatePrivateData(data);
    } catch (err) {
      setError(err as Error);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [client]);

  const updateEmail = useCallback(async (email: string) => {
    setIsLoading(true);
    setError(null);
    try {
      await client.updateEmail(email);
    } catch (err) {
      setError(err as Error);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [client]);

  return {
    isAuthenticated: client.isAuthenticated,
    user: client.user,
    userID: client.userID,
    mustChangePassword: client.mustChangePassword,
    emailVerified: client.emailVerified,
    isLoading,
    error,
    login,
    loginWithToken,
    signup,
    logout,
    changePassword,
    updateProfile,
    updatePrivateData,
    updateEmail,
  };
}
