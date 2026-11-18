# Imagen del chaincode como servicio (CCaaS). Se construye con el CLI de Docker.
FROM golang:1.21-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -o /out/supplycc .

FROM alpine:3.19
COPY --from=build /out/supplycc /usr/local/bin/supplycc
EXPOSE 9999
CMD ["supplycc"]
