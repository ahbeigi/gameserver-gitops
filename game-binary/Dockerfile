# build
FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

# run
FROM gcr.io/distroless/base-debian12
COPY --from=build /out/server /server
USER 65532:65532
ENV GAME_PORT=30000
EXPOSE 30000/tcp
ENTRYPOINT ["/server"]
