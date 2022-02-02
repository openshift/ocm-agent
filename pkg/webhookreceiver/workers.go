package webhookreceiver

import "log"

func processAMReceiver(d AMReceiverData) {
	log.Printf("Process alert data: %+v\n", d)
}
