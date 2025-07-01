# Import with Password
tofu import k3s_server.main "host=192.168.10.1,user=ubuntu,password=$PASS"

# Import with key
tofu import k3s_server.main "host=192.168.10.1,user=ubuntu,private_key=$SSH_KEY"
