cd ../../..
make dashboard
cd profaastinate/deployment/docker
docker-compose up -d --build
./connect-dashboard-docker.sh
