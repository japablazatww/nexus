FROM golang:1.23-alpine

WORKDIR /app

# We will handle the build in the docker-compose context
COPY . .

RUN go build -mod=vendor -o nexus-server ./nexus

CMD ["./nexus-server"]
