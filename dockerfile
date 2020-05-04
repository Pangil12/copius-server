FROM ubuntu
MAINTAINER "Rob"
#Install git
RUN apt-get update \        
     apt-get install -y git
RUN mkdir /home/sampleTest \      
           cd /home/sampleTest \        
           git clone https://github.com/Pangil12/copius-server.git
#Set working directory
WORKDIR /home/sampleTest
