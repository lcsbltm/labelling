FROM golang:1.25-alpine AS build
ARG APP
WORKDIR /app
RUN test -n "${APP}" || (echo "APP is required: api, sync, or worker" >&2; exit 1)
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /app-binary ./cmd/${APP}

FROM scratch
COPY --from=build /app-binary /app
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app"]
