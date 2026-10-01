FROM node:24.8.0-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build
FROM nginxinc/nginx-unprivileged:1.30.3-alpine
LABEL org.opencontainers.image.source="https://github.com/wmcbay13/ticket-platform"
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
USER 101:101
EXPOSE 8080
