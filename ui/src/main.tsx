import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { HeroUIProvider, ToastProvider } from '@heroui/react'
import './index.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <HeroUIProvider locale="zh-CN">
      <App />
      <ToastProvider placement="bottom-right" />
    </HeroUIProvider>
  </StrictMode>,
)
