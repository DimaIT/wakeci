const wsMessageHandler = function(app, data) {
    const msg = JSON.parse(data);
    switch (msg.type) {
    case "jobs:list":
        app.$store.commit("WS_MSG_JOBS_LIST", msg.data);
        break;
    default:
        console.warn("Unhandled message", msg);
    }
};

export default wsMessageHandler;