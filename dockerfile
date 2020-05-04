FROM ubuntu
MAINTAINER "Rob"
#Install git
RUN apt-get update       
RUN apt-get install -y git
RUN mkdir sampleTest
RUN cd sampleTest
RUN git clone https://github.com/Pangil12/copius-server.git
#Set working directory
WORKDIR /home/sampleTest
