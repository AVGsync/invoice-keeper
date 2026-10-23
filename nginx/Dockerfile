FROM nginx:1.27.5-alpine3.21

# Шаблон, а не готовый конфиг: DOMAIN подставляет штатный entrypoint образа
# при каждом старте. NGINX_ENVSUBST_FILTER оставляет в покое переменные самого
# Nginx ($host, $uri и остальные).
ENV NGINX_ENVSUBST_FILTER=DOMAIN
COPY default.conf.template /etc/nginx/templates/default.conf.template

EXPOSE 80 443
