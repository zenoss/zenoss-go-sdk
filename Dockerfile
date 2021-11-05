FROM golang:1.16-alpine3.13 AS build

RUN apk --no-cache --update add gcc git make musl-dev
