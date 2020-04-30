FROM 192.168.56.10:5000/mycentos:v0.0.2
ADD client.tar.gz /home
ADD server.tar.gz /home
COPY start.sh /home
CMD ["sh", "/home/start.sh"]
