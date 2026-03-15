FROM ubuntu
COPY . .
ENV API_KEY=supersecret123
ENV DB_PASSWORD=mypassword456
RUN apt-get update
RUN apt-get install -y curl
RUN apt-get install -y vim
RUN apt-get install -y git

#random test