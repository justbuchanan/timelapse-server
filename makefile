
run:
	mkdir -p example
	mkdir -p out
	go run main.go --image-dir example --out-dir out

pretty:
	go fmt .
	yapf --style Google -i *.py

testloop:
	# this uses the 'filewatcher' ruby gem
	filewatcher ./* 'go test .; echo'
