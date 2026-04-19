FROM docker.io/library/python:3.14.4-alpine3.22

WORKDIR /app
COPY app/server.py /app/server.py

ENV PYTHONUNBUFFERED=1
EXPOSE 8080

CMD ["python", "/app/server.py"]
