FROM python:3.12-slim-bookworm
WORKDIR /app
RUN useradd --create-home --uid 10001 --shell /usr/sbin/nologin app
COPY server.py /app/server.py
COPY static /app/static
USER app
EXPOSE 8090
CMD ["python", "-u", "/app/server.py", "--host", "0.0.0.0", "--port", "8090"]
