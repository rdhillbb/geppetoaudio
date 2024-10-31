graph TD
    subgraph Client["ChatClient"]
        Start["Start()"]
        KA["keepAliveRoutine()"]
        RR["receiveRoutine()"]
        AP["audioProcessingRoutine()"]
        
        subgraph Channels
            MC["MessageChannel"]
            DC["DisplayChannel"]
            AC["AudioChannel"]
            Done["Done Channel"]
        end
    end

    subgraph WebSocket
        WS["WebSocket Connection"]
        Ping["Ping Messages"]
        Pong["Pong Messages"]
        Data["Data Messages"]
    end

    Start -->|"Spawns"| RR
    Start -->|"Reads Input"| UserInput["User Input Handler"]
    
    KA -->|"PingInterval"| Ping
    KA -->|"Monitors"| Done
    
    WS -->|"Sends"| Pong
    WS -->|"Receives"| Data
    
    RR -->|"Processes"| Data
    RR -->|"Routes Audio"| AC
    RR -->|"Monitors"| Done
    
    AC -->|"Feeds"| AP
    AP -->|"Monitors"| Done
    
    Data -->|"Audio Delta"| AudioHandler["Audio Handler"]
    AudioHandler -->|"Chunks"| AC
    
    classDef routine fill:#f9f,stroke:#333,stroke-width:2px,color:#000
    classDef channel fill:#bbf,stroke:#333,stroke-width:2px,color:#000
    classDef ws fill:#bfb,stroke:#333,stroke-width:2px,color:#000
    
    class KA,RR,AP routine
    class MC,DC,AC,Done channel
    class WS,Ping,Pong,Data ws
