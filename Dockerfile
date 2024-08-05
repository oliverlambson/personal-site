FROM python:3.12-alpine

COPY ./web/static/ /app/static/

EXPOSE 1960
CMD ["python", "-m", "http.server", "1960", "--directory", "/app/static/"]

