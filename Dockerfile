FROM justbuchanan/docker-archlinux
MAINTAINER Justin Buchanan <justbuchanan@gmail.com>

RUN pacman -Syyu --noconfirm
RUN pacman -S --noconfirm go python python-numpy opencv hdf5 mencoder
RUN rm /var/cache/pacman/pkg/*

EXPOSE 8888
VOLUME /data
VOLUME /www

RUN mkdir timelapse-server
WORKDIR timelapse-server

COPY image_brightness.py ./
COPY main.go ./

CMD go run main.go --image-dir /data --out-dir /www 
