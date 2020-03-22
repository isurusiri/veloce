# Veloce

<img src="gopher.png" style="display:block;margin-left:auto;margin-right:auto;width:50%" alt="drawing"/>

An experimental in memory key value store written in Go.

Some features that Veloce serves as of now,   
* Thread safe due to the data structure of the cache, i.e. map[string]interface{}
* Possibility of setting a expiration time for items stored in.
* Doesn't need to serialize its contents.
* Any type of a object can be stored and cache can be safely used by multipel goroutines.   

### Installation   

### Usage   

