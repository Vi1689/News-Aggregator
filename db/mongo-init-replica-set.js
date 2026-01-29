# В mongosh выполнить:
rs.initiate({
  _id: "rs0",
  members: [
    { _id: 0, host: "mongodb:27017", priority: 2 },
    { _id: 1, host: "mongodb-secondary-1:27017", priority: 1 },
    { _id: 2, host: "mongodb-secondary-2:27017", priority: 1 }
  ]
})


