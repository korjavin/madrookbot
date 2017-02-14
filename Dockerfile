FROM korjavin/korjavin-base
RUN apt-get update
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y python3-pip python-pip
RUN /usr/bin/pip install --upgrade --user awscli
RUN ln -s /root/.local/bin/aws /bin/aws
RUN mkdir /bot
ADD madrookbot /bot/madrookbot
ADD $HOME/.aws /root/.aws

WORKDIR /bot
ENTRYPOINT ["/bot/madrookbot"]
