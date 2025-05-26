import React, { useState } from "react";

function ChatBox() {
  const [messages, setMessages] = useState([]);
  const [input, setInput] = useState("");

  const sendMessage = async () => {
    const response = await fetch("http://localhost:8080/api/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
     // body: JSON.stringify({ message: input }),
     body: JSON.stringify({
        model: "gpt-4.1-mini",
        messages: [
          { role: "user", content: input }
        ]
      })
    });

    const data = await response.json();
    console.log("Response:", data);
   // setMessages([...messages, { user: input, bot: data.reply }]);
    
    setMessages([...messages, { user: input, bot: data.choices[0].message.content }]);
    setInput("");
  };

  return (
    <div>
      <div style={{ maxHeight: "300px", overflowY: "auto" }}>
        {messages.map((m, idx) => (
          <div key={idx}>
            <strong>You:</strong> {m.user}<br />
            <strong>Bot:</strong> {m.bot}
            <hr />
          </div>
        ))}
      </div>
      <input
        type="text"
        value={input}
        onChange={(e) => setInput(e.target.value)}
      />
      <button onClick={sendMessage}>Send</button>
    </div>
  );
}

export default ChatBox;
