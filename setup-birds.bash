#!/bin/bash

#
# This file sets up a "birds" project folder and generates some of
# the documents needed to build our web application.
#
mkdir -p birds/htdocs

# Generate the empty files we'll use in the demo.
touch birds/setup.sql
touch birds/birds.csv
touch birds/postgrest.conf
touch birds/htdocs/index.html
touch birds/htdocs/sightings.js

# Create the database we'll use in the demo.
if ! createdb birds 2>/dev/null; then
	dropdb birds
	createdb birds
fi

# Generate a README
cat <<EOT>birds/README.md

# Birds, a demo of PostgreSQL 15, PostgREST 11

This directory holds our demo.

EOT

# Generate our SQL setup modeling our simple data
cat <<EOT>birds/setup.sql
-- Make sure we're in the birds database
\c birds

DROP SCHEMA IF EXISTS birds CASCADE;
CREATE SCHEMA birds;
SET search_path TO birds, public;

--
-- This file contains the create statements for our bird table.
--
-- DROP TABLE IF EXISTS birds.sighting;
CREATE TABLE birds.sighting
(
  bird_name VARCHAR(255),
  place TEXT,
  sighted DATE
);

-- bird_view will become an end point in PostgREST
CREATE OR REPLACE VIEW birds.bird_view AS
  SELECT bird_name AS bird, trim(place) AS place, sighted
  FROM birds.sighting ORDER BY sighted ASC, bird_name ASC;

-- record_bird is a stored procedure and will save a new bird sighting
CREATE OR REPLACE FUNCTION birds.record_bird(bird VARCHAR, place TEXT, sighted DATE)
RETURNS bool LANGUAGE SQL AS \$\$
  INSERT INTO birds.sighting (bird_name, place, sighted) VALUES (bird, place, sighted);
  SELECT true;
\$\$;

--
-- The following additional steps are needed for PostgREST access.
--
DROP ROLE IF EXISTS birds_anonymous;
CREATE ROLE birds_anonymous nologin;

GRANT USAGE  ON SCHEMA birds      TO birds_anonymous;
-- NOTE: We're allowing insert because this is a demo and we're not
-- implementing a login requirement!!!!
GRANT SELECT, INSERT ON birds.sighting    TO birds_anonymous;
GRANT SELECT ON birds.bird_view   TO birds_anonymous;
GRANT EXECUTE ON FUNCTION birds.record_bird TO birds_anonymous;

DROP ROLE IF EXISTS birds;
CREATE ROLE birds NOINHERIT LOGIN PASSWORD 'my_secret_password';
GRANT birds_anonymous TO birds;

EOT

# Generate some test data to load into our models
cat <<EOT>birds/birds.csv
bird,place,sighted
robin, seen in my backyard,2023-04-16
humming bird, seen in my backyard, 2023-02-28
blue jay, seen on my back porch, 2023-01-12
EOT

# Generate a template of postgrest.conf file.
cat <<EOT>birds/postgrest.conf
db-uri = "postgres://birds:my_secret_password@localhost:5432/birds"
db-schemas = "birds"
db-anon-role = "birds_anonymous"
EOT

# Generate index.html
cat <<EOT>birds/htdocs/index.html
<DOCTYPE html lang="en">
<html>
  <body>
    <h1>Welcome to the bird list!</h1>
    <div id="bird-list"></div>
    <h2>Add a bird</h2>
    <div><form>
      <div>
        <label for="bird">Bird</label>
		<input id="bird" id="bird" type="text" value="">
      </div>
      <div>
        <label for="place">Place</label>
		<input id="place" id="place" type="text" value="">
      </div>
      <div>
        <label for="sighted">Sighted on</label>
		<input id="sighted" id="sighted" type="date">
      </div>
      <button id="record-bird" type="submit">Add Bird Sighting</button>
    </form></div>
    <script src="sightings.js"></script>
  </body>
</html>
EOT

# Generate sightings.js
cat <<EOT>birds/htdocs/sightings.js
/* sightings.js provides access to our JSON data API run by PostgREST
   and assembles the results before updating the web page. */
(function(document, window) {
  let list_url = 'http://localhost:3000/bird_view',
    record_url = 'http://localhost:3000/rpc/record_bird',
    list_elem = document.getElementById('bird-list'),
    bird_elem = document.getElementById('bird'),
    place_elem = document.getElementById('place'),
    sighted_elem = document.getElementById('sighted'),
    add_button = document.getElementById('record-bird');

  function updateList(elem, src) {
    let bird_list = JSON.parse(src),
      parts = [];
    parts.push('<table>');
    parts.push('  <tr><th>bird</th><th>place</th><th>sighted</th></tr>');
    for (const obj of bird_list) {
      parts.push(\` <tr><td>\${obj.bird}</td><td>\${obj.place}</td><td>\${obj.sighted}</td></td>\`);
    }
    parts.push('</table>');
    elem.innerHTML = parts.join('\n');
  }

  function birdRecord(bird_elem, place_elem, sighted_elem) {
    return { "bird": bird_elem.value, "place": place_elem.value, "sighted": sighted_elem.value };
  }

  function getData(elem, url, updateFn) {
    /* We use a xhr to retrieve the current list of sightings. */
    const req = new XMLHttpRequest();
    req.addEventListener("load", function(evt) {
      /* Call our page update function */
      updateFn(elem, this.responseText);
    });
    req.open("GET", url);
	req.setRequestHeader('Cache-Control', 'no-cache');
    req.send();
  };

  function postData(obj, url) {
    const req = new XMLHttpRequest();
    req.open("POST", url, true);
    req.setRequestHeader('Content-Type', 'application/json');
    req.onreadystatechange = function() {//Call a function when the state changes.
      console.log(\`DEBUG state \${req.readyState}, status \${req.status}, resp \${req.responseText}\`);
    }
    req.send(JSON.stringify(obj));
  }

  /* Main processing for page */
  add_button.addEventListener("click", function(evt) {
    postData(birdRecord(bird_elem, place_elem, sighted_elem), record_url);
	/* Now we need to update our listing! */
	list_elem.innerHTML = '';
	setTimeout(() => {
  		console.log("Delayed for 10 second.");
  		getData(list_elem, list_url, updateList);
	}, "10 second");
    evt.preventDefault();
  });

  getData(list_elem, list_url, updateList);
})(document, window);
EOT
