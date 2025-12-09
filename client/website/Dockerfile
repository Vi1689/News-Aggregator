FROM nginx:alpine

WORKDIR /usr/share/nginx/html

COPY nginx.conf /etc/nginx/nginx.conf

COPY . .

RUN echo "Frontend is healthy" > /healthcheck.html

EXPOSE 8888

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8888/healthcheck.html || exit 1

CMD ["nginx", "-g", "daemon off;"]