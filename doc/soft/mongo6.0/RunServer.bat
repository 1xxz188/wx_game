if not exist data (
       mkdir data
)

if not exist "data/s0_0" (
       mkdir "data/s0_0"
)

if not exist "data/log" (
       mkdir "data/log"
)

start "Mongo-S0_0" mongod.exe --dbpath ./data/s0_0 --logpath ./data/log/s0_0.log --port 26001 --bind_ip 0.0.0.0 --wiredTigerCacheSizeGB 1  --journal --logappend
