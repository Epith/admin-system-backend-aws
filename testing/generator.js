const variables = {
    latMax: 38.735083,
    latMin: 40.898677,
    lngMax: -77.109339,
    lngMin: -81.587841
  }
  
  const generateRandomData = (userContext, events, done) => {
    userContext.vars.userId = "545bc2aa-7a37-11ee-b962-0242ac120002"
    userContext.vars.pointsId = "points uuid"
    userContext.vars.reqId = "req uuid"
  
    return done()
  }