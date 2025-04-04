from pathlib import Path

NUM_SHARDS = 3
PATH_REPOSITORY = Path(__file__).parent.parent
PATH_ROSETTA = PATH_REPOSITORY / "cmd" / "rosetta" / "rosetta"
PATH_PROXY_TO_OBSERVER_ADAPTER = PATH_REPOSITORY / "systemtests" / "proxyToObserverAdapter"
ADAPTER_DELAY_IN_MILLISECONDS = 0
ADDITIONAL_GAS_LIMIT_FOR_RELAYED_V3 = 50_000
AWAITING_POLLING_TIMEOUT_IN_MILLISECONDS = 1000
AWAITING_PATIENCE_IN_MILLISECONDS = 0

METACHAIN_ID = 4294967295
SHARDS = [0, 1, 2, METACHAIN_ID]
