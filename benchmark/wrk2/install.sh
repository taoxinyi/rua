sudo apt-get install -y libz-dev build-essential libssl-dev
git clone https://github.com/giltene/wrk2
cd wrk2
make
sudo ln -s $PWD/wrk /usr/bin/wrk2
cd ~
