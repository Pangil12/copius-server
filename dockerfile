FROM ubuntu
MAINTAINER "Rob"
#Install git
RUN apt-get update       
RUN apt-get install -y git
RUN cd /home/sampleTest        
RUN git clone https://github.com/Pangil12/copius-server.git
#Set working directory
WORKDIR /home/sampleTest
