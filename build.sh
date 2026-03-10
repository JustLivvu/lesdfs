cd client 
go build -o lesd 
cd .. 
cd daemon 
go build -o lesd-daemon
cd ..
echo -e "\033[92midk probably built succesfully ok.\033[0m"
read -p "do you want to copy client to ~/.local/bin? (y/n): " answer

if [[ "$answer" == "y" || "$answer" == "Y" ]]; then
    cd client
    mv lesd ~/.local/bin/
    cd ..
else
    echo "ok"
    cd ..
fi
read -p "do you want to copy daemon to ~/.local/bin? (y/n): " answer

if [[ "$answer" == "y" || "$answer" == "Y" ]]; then
    cd daemon
    mv lesd-daemon ~/.local/bin/
    cd ..
else
    echo "ok"
    cd ..
fi