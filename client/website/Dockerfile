FROM nginx:alpine

WORKDIR /usr/share/nginx/html

COPY nginx.conf /etc/nginx/nginx.conf

COPY . .

RUN echo "Frontend is healthy" > /healthcheck.html

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost/healthcheck.html || exit 1

CMD ["nginx", "-g", "daemon off;"]