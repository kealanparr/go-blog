# Aborts our script if there's an error
set -e

# Word words words
echo "Beginning build proccess.."
docker-compose up -d

# Build & run golang
go build main.go
./main