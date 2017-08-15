FROM justbuchanan/docker-archlinux
MAINTAINER Justin Buchanan <justbuchanan@gmail.com>

RUN pacman -Syyu --noconfirm
RUN pacman -S --noconfirm go python python-numpy opencv hdf5 mencoder git -v
RUN rm /var/cache/pacman/pkg/*

ENV GOPATH=/go
RUN go get -u github.com/pkg/errors

EXPOSE 8888
VOLUME /data
VOLUME /www

RUN mkdir timelapse-server
WORKDIR timelapse-server

COPY image_brightness.py ./
COPY main.go ./

# West coast
# TODO: pull this from the host system?
# Borrowed from: https://serverfault.com/questions/683605/docker-container-time-timezone-will-not-reflect-changes
ENV TZ=America/Los_Angeles
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

RUN go build -o timelapse-server main.go
CMD ./timelapse-server --image-dir /data --out-dir /www 
