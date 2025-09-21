# Fintech Ledger Service

A distributed ledger service for financial transactions with multi-tenant support.

## Features

- Multi-tenant architecture with schema isolation
- Event sourcing for audit trails
- RESTful API with proper error handling
- Database migrations
- Docker support for development

## Quick Start

```bash
# Install dependencies
make deps

# Setup environment
make setup

# Run with Docker
docker-compose up -d

# Start the application
make run
```

## API Documentation

See [docs/api.md](docs/api.md) for detailed API documentation.

## Development

```bash
# Run tests
make test

# Format code
make fmt

# Lint code
make lint
```

## Deployment

The application can be containerized using the provided Dockerfile:

```bash
docker build -t ledger-service .
docker run -p 8080:8080 ledger-service
```
