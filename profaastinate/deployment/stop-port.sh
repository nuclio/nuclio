docker-compose down  # Stop container on current dir if there is a docker-compose.yml
docker rm -fv $(docker ps -aq)  # Remove all containers
sudo lsof -i -P -n | grep 5000  # List who's using the port
sudo kill -9 `sudo lsof -t -i:5000`