FROM python:3.12-alpine

WORKDIR /app
COPY app/server.py /app/server.py

ENV PYTHONUNBUFFERED=1
EXPOSE 8080

CMD ["python", "/app/server.py"]
