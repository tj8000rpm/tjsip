package main

import (
	"sip/sip"
)

func messageManipulationHandler(layer int, srv *sip.Server, msg *sip.Message) error {
	// if layer != sip.LayerParserIngress || layer != sip.LayerParserEgress {
	// 	// return fmt.Errorf("Error: %s", "Unexpected calling")
	// 	return nil
	// }

	return nil
}
