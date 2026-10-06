import { BrowserRouter } from 'react-router-dom';
import { AuthProvider } from './context/AuthContext';
import { ConsentProvider } from './context/ConsentContext';
import { ToastProvider, ToastViewport } from './context/ToastContext';
import { ThemeProvider } from './context/ThemeContext';
import AppRouter from './router/AppRouter';

export default function App() {
  return (
    <ThemeProvider>
      <BrowserRouter>
        <ToastProvider>
          <AuthProvider>
            <ConsentProvider>
              <AppRouter />
              <ToastViewport />
            </ConsentProvider>
          </AuthProvider>
        </ToastProvider>
      </BrowserRouter>
    </ThemeProvider>
  );
}