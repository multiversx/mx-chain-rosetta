from pathlib import Path

PATH_REPOSITORY = Path(__file__).parent.parent
PATH_ROSETTA = PATH_REPOSITORY / "cmd" / "rosetta" / "rosetta"
PATH_PROXY_TO_OBSERVER_ADAPTER = PATH_REPOSITORY / "systemtests" / "proxyToObserverAdapter"
PORT_ROSETTA = 7091
PORT_OBSERVER_SURROGATE = 8080
ADAPTER_DELAY_IN_MILLISECONDS = 100
