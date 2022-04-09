# unitehud
Pokemon Unite scoreboard HUD and extra tools running over captured game feeds using OpenCV with a Gio interface.

### Note
##### This project is currently in early Alpha stages. 
##### It would be possible for matching techniques to produce duplicated, unaccounted-for, and false postitive matches.
##### Winner/Loser confidence is successful ~99% of the time.
##### Score tracking is ~90% accurate, certain game mechanics (like rotom scoring points) are extremely difficult to process.
##### Users are encouraged to report issues, or contribute where they can to help polish a final product.

----

### Client (OBS Live)
![alt text](https://github.com/pidgy/unite/blob/master/data/client.gif "Client")

### Server
![alt text](https://i.imgur.com/pR525QW.png "server")

### Architecture

- The server opens port 17069 by default as a Websocket and HTTP endpoint. 
- The client sends a GET request every second to the server and updates it's page.

#### Client Request
```
GET 127.0.0.1:17069/http
```

#### Server Response
```
{
    "orange": {
        "team": "orange",
        "value": 52
    },
    "purple": {
        "team": "purple",
        "value": 46
    },
    "seconds": 389,
    "self": {
        "team": "self",
        "value": 0
    }
}
```

- Wiki for tutorial's
