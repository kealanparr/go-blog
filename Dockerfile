FROM postgres
COPY db/init.sql /docker-entrypoint-initdb.d/init.sql
COPY db/seedData.sql /docker-entrypoint-initdb.d/seedData.sql