FROM node:latest
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 3000
# สั่งรันด้วย Vite บนพอร์ต 3000 และเปิดให้เข้าถึงจากภายนอก Container ได้ (--host)
CMD ["npm", "run", "dev", "--", "--port", "3000", "--host"]
