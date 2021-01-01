sudo apt-get install -y build-essential libssl-dev git
git clone https://github.com/wg/wrk
cd wrk
make
sudo ln -s $PWD/wrk /usr/bin/
cd ~
