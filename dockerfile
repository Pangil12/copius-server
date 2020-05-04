FROM ubuntu
MAINTAINER "Rob"
#Install git
RUN sudo apt-get update       
RUN apt-get install -y git
RUN sudo mkdir /home/sampleTest \      
           cd /home/sampleTest \        
           sudo git clone https://github.com/Pangil12/copius-server.git
#Set working directory
WORKDIR /home/sampleTest
