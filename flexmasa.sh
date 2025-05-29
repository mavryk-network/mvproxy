image=mavrykdynamics/flexmasa:latest
script=boreasbox
docker run --rm --name my-sandbox --detach -p 8732:20000 \
       -e block_time=3 \
       "$image" "$script" start
