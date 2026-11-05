package actions

/*
1. Process cluster templates
2. Generate cluster names
3. Track ClusterIDs in ClusterState file
  - Must be synchronized (mutex)
4. Check for existence of clusters tracked in ClusterState before creation (idempotency)
5.
*/
