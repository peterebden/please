FROM golang:1.10

env PLZ_ARGS "-o build.path:/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin"

RUN apt-get update && apt-get install -y unzip patch && apt-get clean
