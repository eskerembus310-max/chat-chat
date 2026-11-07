FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /chatchat .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /chatchat /chatchat
ENV DATA_DIR=/data
EXPOSE 10000
CMD ["/chatchat"]
