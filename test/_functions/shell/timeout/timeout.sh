export EVENT_BODY=$(cat)

if [ "$(echo ${EVENT_BODY} | cut -d' ' -f1)" == "sleep" ]; then
  export sleepTimeout=$(echo ${EVENT_BODY} | cut -d' ' -f2)
	sleep ${sleepTimeout}
	exit 0
fi

# doing nothing while waiting for processor to kill me
while true; do
  read
done < /dev/stdin
