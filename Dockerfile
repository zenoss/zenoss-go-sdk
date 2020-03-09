FROM golang:1.14.1-alpine AS build

RUN apk --no-cache --update add gcc git make musl-dev
