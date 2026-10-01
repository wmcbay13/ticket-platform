import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
export default defineConfig({plugins:[react()],server:{proxy:{'/api/events':'http://localhost:8081','/api/orders':'http://localhost:8082'}}})
