
run:
	mkdir -p example
	mkdir -p out
	go run main.go --image-dir example --out-dir out

dockerbuild:
	docker build -t img .

dockerrun: dockerbuild
	mkdir -p example
	mkdir -p out
	docker run -v /home/justin/go/src/github.com/justbuchanan/timelapse-server/example:/data -v /home/justin/go/src/github.com/justbuchanan/timelapse-server/out:/www -t img

pretty:
	go fmt .
	yapf --style Google -i *.py

testloop:
	# this uses the 'filewatcher' ruby gem
	filewatcher ./* 'make tests; echo'

tests:
	go test .
