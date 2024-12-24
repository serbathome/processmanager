1. when starting read parameters
2. start processes aefuf and aefad, they will listen on 127.0.0.1 to ports 2888 and 3888 accordinglyå
3. regularly check if started processed are healty and listening to network ports
4. if one or both processes are not running or not listening to ports, restart them
5. create health check http endpoint, which if called will try to establish tcp connection to ports which aefuf and aefad are listening. If can't establlish connection, fail the probe. Otherwise return 200.
6. create restart endpoint, which if called restarts aefuf and aefad processes. If restart is called, but processes are ok, don't restart.