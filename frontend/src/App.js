import React from "react";
import ChatBox from "./components/ChatBox";

function App() {
    const authUrl = `https://auth.atlassian.com/authorize?audience=api.atlassian.com&client_id=${CLIENT_ID}&scope=read:confluence-content.all&redirect_uri=${REDIRECT_URI}&response_type=code&prompt=consent`;

window.location.href = authUrl;

  return (
    <div>
      <h1>Help Chatbot</h1>
      <ChatBox />
    </div>
  );
}

export default App;
