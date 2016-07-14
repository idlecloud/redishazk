# redis_ha compile script
#
# CJP

all: redis_ha
	echo "SUCCESS"

redis_ha: redis_ha.go
	godep go build redis_ha.go

clean:
	rm -rf redis_ha
