describe('The Home Page', function() {
  it('successfully loads', function() {
    cy.visit('/');
    cy.request({
      method: 'POST',
      url: '/api/login',
      form: true, // indicates the body should be form urlencoded and sets Content-Type: application/x-www-form-urlencoded headers
      body: {
        username: 'leonardo',
        password: 'leonardo'
      }
    }).then((resp) => {
      expect(resp.status).to.eq(200);
    });
    cy.get('#username').type('leonardo');
    cy.get('#password').type('leonardo');
    cy.get('[data-cy=submit]').click().as('comments');
  });

  // it('dashboard', function() {
  //   cy.get('[data-cy=register]').should('contain', 'Registered Devices');
  //   cy.get('[data-cy=registerNumber]').should('contain', '2');
  // })
});
