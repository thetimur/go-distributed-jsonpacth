import * as React from "react"
import * as ReactDOM from "react-dom"
import { createEvent, createStore } from "effector"
import { useEvent, useStore } from "effector-react"

import { ConstantBackoff, Websocket, WebsocketBuilder, WebsocketEvent } from "websocket-ts"

import { applyPatch } from "fast-json-patch"

const patchSnapEv = createEvent()
const $snap = createStore<any>({})
$snap.on(patchSnapEv, (state, patch:any) => {
  const doc = applyPatch(state, patch, false, false).newDocument
  return doc
})

const buttonStyle = {
  padding: '5px 10px',
  margin: '5px',
  cursor: 'pointer',
};


const ws = new WebsocketBuilder("ws://localhost:8080/ws")
  .withBackoff(new ConstantBackoff(10*1000))
  .build()

const receiveMessage = (i: Websocket, ev: MessageEvent) => {
  const transaction = JSON.parse(ev.data)
  const patch = JSON.parse(transaction.Payload)
  patchSnapEv(patch)
}

const setPlayerNameEv = createEvent<string>()
const $playerName = createStore<string>("")
$playerName.on(setPlayerNameEv, (_, payload) => {
  return payload
})

function PlayerInput() {
    const setPlayerName = useEvent(setPlayerNameEv)
    const playerName = useStore($playerName)
    const handleChange = function(event: React.ChangeEvent<HTMLInputElement>) {
      setPlayerName(event.target.value)
    }
  
    const snap = useStore($snap)
    
    const handleClick = function() {
      fetch("http://127.0.0.1:8080/replace", {
        method: "POST",
        body:`[{"op":"add", "path": "/${playerName}", "value": {"x":20, "y":20}}]`})
    }
  
    const updatePlayer = function(player:any) {
      fetch("http://127.0.0.1:8080/replace", {
        method: "POST",
        body:`[{"op":"add", "path": "/${playerName}", "value": {"x":${player.x}, "y":${player.y}}}]`})
    }
    const handleUp = function() {
      let player = snap[playerName]
      player.y--
      updatePlayer(player)
    }
  
    const handleDown = function() {
      let player = snap[playerName]
      player.y++
      updatePlayer(player)
    }
  
    const handleLeft = function() {
      let player = snap[playerName]
      player.x--
      updatePlayer(player)
    }
  
    const handleRight = function() {
      let player = snap[playerName]
      player.x++
      updatePlayer(player)
    }
  
    return (<>
      <input type="text" style={buttonStyle} 
        onChange={handleChange}
        value={playerName}/>
      <input type="button" style={buttonStyle} 
        value={"Установить"}
        onClick={handleClick}/>
      <input type="button" style={buttonStyle} 
        value={"Вверх"}
        onClick={handleUp}/>
      <input type="button" style={buttonStyle} 
        value={"Вниз"}
        onClick={handleDown}/>
      <input type="button" style={buttonStyle} 
        value={"Влево"}
        onClick={handleLeft}/>
      <input type="button" style={buttonStyle} 
        value={"Вправо"}
        onClick={handleRight}/>
    </>)
  }

ws.addEventListener(WebsocketEvent.message, receiveMessage)

function Grid({ width, height, step }: { width: number, height: number, step: number }) {
  const lines = [];
  for (let i = 0; i <= width; i += step) {
    lines.push(<line x1={i} y1="0" x2={i} y2={height} stroke="#ddd" />);
  }
  for (let j = 0; j <= height; j += step) {
    lines.push(<line x1="0" y1={j} x2={width} y2={j} stroke="#ddd" />);
  }
  return <>{lines}</>;
}


function Players() {
  const snap = useStore($snap);
  let players = [];
  const playerSize = 40;

  for (const k in snap) {
    let player = snap[k];
    players.push(
      <image
        href="jet-pack-jack-thrusters-on.svg"
        x={player.x - playerSize / 2}
        y={player.y - playerSize / 2}
        width={playerSize}
        height={playerSize}
      />
    );
  }
  return <>{players}</>;
}


ReactDOM.render(
    <React.StrictMode>
      <PlayerInput></PlayerInput>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 600">      
        <Grid width={800} height={600} step={20} />
        <Players></Players>
      </svg>
    </React.StrictMode>,
    document.getElementById("root")
  )
  