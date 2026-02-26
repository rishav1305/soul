FROM node:20
WORKDIR /app
COPY . .
RUN npm install
USER root
EXPOSE 3000
CMD ["node", "server.js"]
