FROM postgres:15.3-alpine
WORKDIR /var
RUN apk update && \
    apk add jq wget
RUN mkdir /var/migrations && \
# Create a list of download URLs for migration files
    wget -q -O - https://api.github.com/repos/open-sauced/api/contents/migrations \
    | jq --raw-output '.[].download_url' \
    > ./original_urls.txt
# Downloads all SQL files into /var/migrations
RUN wget -q -P /var/migrations -i ./original_urls.txt && \
    cat /var/migrations/*.sql > /var/all-migrations.sql
# Rename password from environment and run file
CMD PGPASSWORD=${POSTGRES_PASSWORD} psql \
    --host db \
    --username postgres \
    --dbname postgres \
    -f /var/all-migrations.sql
