import React, { useState } from "react";

function AgentBox() {
  const [messages, setMessages] = useState([]);
  const [input, setInput] = useState("");
  const [agentName, setAgentName] = useState("");
  const [agentIns, setAgentIns] = useState("");
  const [file, setFile] = useState(null);

  const sendMessage = async () => {
    const response = await fetch("http://localhost:8080/api/agent", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        model: "gpt-4.1-mini",
        messages: [
          { role: "user", content: input }
        ]
      })
    });

    const data = await response.json();
    console.log("Response:", data);

    setMessages([...messages, { user: input, bot: data.choices[0].message.content }]);
    setInput("");
  };

  const createAgent = async () => {
    const response = await fetch("http://localhost:8080/assistant", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: agentName, instructions: agentIns, model: "gpt-4o" })
    });

    const data = await response.json();
    console.log("Agent Created:", data);
    setAgentName("");
  };

  const uploadFile = async () => {
    const formData = new FormData();
    formData.append("file", file);

    const response = await fetch("http://localhost:8080/upload", {
      method: "POST",
      body: formData
    });

    const data = await response.json();
    console.log("File Uploaded:", data);
    setFile(null);
  };

  return (
    <div>
      <div>
        <input
          type="text"
          placeholder="Agent Name"
          value={agentName}
          onChange={(e) => setAgentName(e.target.value)}
        />
        <input
          type="text"
          placeholder="Agent Instructions"
          value={agentIns}
          onChange={(e) => setAgentIns(e.target.value)}
        />
        <button onClick={createAgent}>Create Agent</button>
      </div>
      <div>
        <input
          type="file"
          onChange={(e) => setFile(e.target.files[0])}
        />
        <button onClick={uploadFile} disabled={!file}>Upload File</button>
      </div>
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

export default AgentBox;
